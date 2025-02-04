package server

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/go-mysql-org/go-mysql/test_util/test_keys"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/proxy"
)

type Server struct {
	port string

	authProvider *RemoteThrottleProvider

	connectionCfgPerRealm map[uint32]*gomysql.Config
}

func NewServer(port, user, pass string, realmsConnectionStrings map[uint32]string) *Server {
	remoteProvider := &RemoteThrottleProvider{server.NewInMemoryProvider()}
	remoteProvider.AddUser(user, pass)

	cfgs := map[uint32]*gomysql.Config{}

	for u, s := range realmsConnectionStrings {
		cfg, err := gomysql.ParseDSN(s)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to parse dsn")
		}

		cfgs[u] = cfg
	}

	return &Server{
		port:                  port,
		authProvider:          remoteProvider,
		connectionCfgPerRealm: cfgs,
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	l, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	var tlsConf = server.NewServerTLSConfig(test_keys.CaPem, test_keys.CertPem, test_keys.KeyPem, tls.VerifyClientCertIfGiven)

	for connectionID := 0; ; connectionID++ {
		c, err := l.Accept()
		if err != nil {
			return err
		}

		go func(ctx context.Context, connID int) {
			connectionByRealm := map[uint32]proxy.DBConnection{}
			for r, config := range s.connectionCfgPerRealm {
				cc, err := client.Connect(config.Addr, config.User, config.Passwd, config.DBName)
				if err != nil {
					log.Fatal().Err(err).Uint32("realmID", r).Msg("failed to connect to server")
				}

				connectionByRealm[r] = proxy.NewMySQLConnWrapper(cc)
			}

			svr := server.NewServer("8.4.4", mysql.DEFAULT_COLLATION_ID, mysql.AUTH_CACHING_SHA2_PASSWORD, test_keys.PubPem, tlsConf)
			conn, err := server.NewCustomizedConn(c, svr, s.authProvider, proxy.NewTC9Proxy(connID, connectionByRealm, parser.New()))
			if err != nil {
				log.Fatal().Err(err).Int("connID", connID).Msg("failed to create customized connection")
			}

			for {
				err = conn.HandleCommand()
				if err != nil {
					log.Err(err).Msg("could not handle command")
					return
				}
			}
		}(ctx, connectionID)
	}
}

type RemoteThrottleProvider struct {
	*server.InMemoryProvider
}

func (m *RemoteThrottleProvider) GetCredential(username string) (password string, found bool, err error) {
	time.Sleep(time.Millisecond * time.Duration(1))
	return m.InMemoryProvider.GetCredential(username)
}
