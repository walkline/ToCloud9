package proxy

import (
	"fmt"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/sqlparser"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type MockDBConnection struct {
	mock.Mock
}

func (m *MockDBConnection) Execute(query string, args ...interface{}) (*mysql.Result, error) {
	res := m.Called(query, args)
	return res.Get(0).(*mysql.Result), res.Error(1)
}

func (m *MockDBConnection) Prepare(query string) (DBStatement, error) {
	args := m.Called(query)
	return args.Get(0).(DBStatement), args.Error(1)
}

func (m *MockDBConnection) FieldList(table, fieldWildcard string) ([]*mysql.Field, error) {
	res := m.Called(table, fieldWildcard)
	return res.Get(0).([]*mysql.Field), res.Error(1)
}

type MockDBStatement struct {
	mock.Mock
}

func (m *MockDBStatement) Execute(args ...interface{}) (*mysql.Result, error) {
	call := m.Called(args)
	return call.Get(0).(*mysql.Result), call.Error(1)
}

func (m *MockDBStatement) Close() error {
	return m.Called().Error(0)
}

func (m *MockDBStatement) ParamNum() int {
	return 1
}

func (m *MockDBStatement) ColumnNum() int {
	return 1
}

func TestHandleStmtExecute_QueriesRouting(t *testing.T) {
	lowGUID := 42
	parser := parser.New()

	testCases := []struct {
		query          string
		args           []interface{}
		mocks          func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement)
		validateResult func(*mysql.Result)
	}{
		{
			query: "SELECT * FROM characters WHERE guid = ?",
			args:  []interface{}{guid.NewCrossrealmPlayerGUID(2, guid.LowType(lowGUID)).GetRawValue()},
			mocks: func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement) {
				mockConn1 := new(MockDBConnection)
				mockConn2 := new(MockDBConnection)
				mockStmt1 := new(MockDBStatement)
				mockStmt2 := new(MockDBStatement)

				mockConn1.On("Prepare", mock.Anything).Return(mockStmt1, nil)
				mockConn2.On("Prepare", mock.Anything).Return(mockStmt2, nil)
				mockStmt2.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{}, nil)

				return mockConn1, mockConn2, mockStmt1, mockStmt2
			},
		},
		{
			query: `SELECT guid, account, name, race, class, gender, level, xp, money, skin, face, hairStyle, hairColor, facialStyle, bankSlots, restState, playerFlags,
              position_x, position_y, position_z, map, orientation, taximask, cinematic, totaltime, leveltime, rest_bonus, logout_time, is_logout_resting, resettalents_cost,
              resettalents_time, trans_x, trans_y, trans_z, trans_o, transguid, extra_flags, stable_slots, at_login, zone, online, death_expire_time, taxi_path, instance_mode_mask,
              arenaPoints, totalHonorPoints, todayHonorPoints, yesterdayHonorPoints, totalKills, todayKills, yesterdayKills, chosenTitle, knownCurrencies, watchedFaction, drunk,
              health, power1, power2, power3, power4, power5, power6, power7, instance_id, talentGroupsCount, activeTalentGroup, exploredZones, equipmentCache, ammoId,
              knownTitles, actionBars, grantableLevels, innTriggerId, extraBonusTalentCount, UNIX_TIMESTAMP(creation_date) FROM characters WHERE guid = ?`,
			args: []interface{}{guid.NewCrossrealmPlayerGUID(2, guid.LowType(lowGUID)).GetRawValue()},
			mocks: func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement) {
				mockConn1 := new(MockDBConnection)
				mockConn2 := new(MockDBConnection)
				mockStmt1 := new(MockDBStatement)
				mockStmt2 := new(MockDBStatement)

				mockConn1.On("Prepare", mock.Anything).Return(mockStmt1, nil)
				mockConn2.On("Prepare", mock.Anything).Return(mockStmt2, nil)
				mockStmt2.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{
					Resultset: &mysql.Resultset{
						Values: [][]mysql.FieldValue{
							{mysql.NewFieldValue(mysql.FieldValueTypeNull, 0, nil)},
						},
					},
				}, nil)

				return mockConn1, mockConn2, mockStmt1, mockStmt2
			},
			validateResult: func(r *mysql.Result) {
				rawGUID := guid.NewCrossrealmPlayerGUID(2, guid.LowType(lowGUID)).GetRawValue()
				assert.Equal(t, mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, rawGUID, []byte(fmt.Sprintf("%d", rawGUID))), r.Values[0][0])
			},
		},
		{
			query: "SELECT casterGuid, itemGuid, spell, effectMask, recalculateMask, stackCount, amount0, amount1, amount2, base_amount0, base_amount1, base_amount2, maxDuration, remainTime, remainCharges FROM character_aura WHERE guid = ?",
			args:  []interface{}{guid.NewCrossrealmPlayerGUID(2, guid.LowType(lowGUID)).GetRawValue()},
			mocks: func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement) {
				mockConn1 := new(MockDBConnection)
				mockConn2 := new(MockDBConnection)
				mockStmt1 := new(MockDBStatement)
				mockStmt2 := new(MockDBStatement)

				mockConn1.On("Prepare", mock.Anything).Return(mockStmt1, nil)
				mockConn2.On("Prepare", mock.Anything).Return(mockStmt2, nil)
				mockStmt2.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{}, nil)

				return mockConn1, mockConn2, mockStmt1, mockStmt2
			},
		},
		{
			query: "SELECT spell, specMask FROM character_spell WHERE guid = ?",
			args:  []interface{}{guid.NewCrossrealmPlayerGUID(1, guid.LowType(lowGUID)).GetRawValue()},
			mocks: func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement) {
				mockConn1 := new(MockDBConnection)
				mockConn2 := new(MockDBConnection)
				mockStmt1 := new(MockDBStatement)
				mockStmt2 := new(MockDBStatement)

				mockConn1.On("Prepare", mock.Anything).Return(mockStmt1, nil)
				mockStmt1.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{}, nil)
				mockConn2.On("Prepare", mock.Anything).Return(mockStmt2, nil)

				return mockConn1, mockConn2, mockStmt1, mockStmt2
			},
		},
		{
			query: `SELECT creatorGuid, giftCreatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text, bag, slot, item, itemEntry 
              FROM character_inventory ci JOIN item_instance ii ON ci.item = ii.guid WHERE ci.guid = ? ORDER BY bag, slot`,
			args: []interface{}{guid.NewCrossrealmPlayerGUID(2, guid.LowType(lowGUID)).GetRawValue()},
			mocks: func() (*MockDBConnection, *MockDBConnection, *MockDBStatement, *MockDBStatement) {
				mockConn1 := new(MockDBConnection)
				mockConn2 := new(MockDBConnection)
				mockStmt1 := new(MockDBStatement)
				mockStmt2 := new(MockDBStatement)

				mockConn1.On("Prepare", mock.Anything).Return(mockStmt1, nil)
				mockConn2.On("Prepare", mock.Anything).Return(mockStmt2, nil)
				mockStmt2.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{}, nil)

				return mockConn1, mockConn2, mockStmt1, mockStmt2
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.query, func(t *testing.T) {
			mockConn1, mockConn2, mockStmt1, mockStmt2 := tt.mocks()

			connections := map[uint32]DBConnection{
				1: mockConn1,
				2: mockConn2,
			}
			proxyInstance := NewTC9Proxy(1, connections, parser)

			_, _, stmtCtx, err := proxyInstance.HandleStmtPrepare(tt.query)
			assert.NoError(t, err)

			res, err := proxyInstance.HandleStmtExecute(stmtCtx, tt.query, tt.args)
			assert.NoError(t, err)
			if tt.validateResult != nil {
				tt.validateResult(res)
			}

			mockStmt1.AssertExpectations(t)
			mockConn1.AssertExpectations(t)
			mockStmt2.AssertExpectations(t)
			mockConn2.AssertExpectations(t)
		})
	}
}

func TestHandleStmtExecute_StartingTransaction(t *testing.T) {
	lowGUID := 42
	mockConn1 := new(MockDBConnection)
	mockStmt1 := new(MockDBStatement)

	mockConn1.On("Execute", mock.MatchedBy(func(v string) bool { return v == StartTransaction }), mock.Anything).Return(&mysql.Result{}, nil)
	mockStmt1.On("Execute", mock.MatchedBy(func(v []interface{}) bool { return v[0].(uint32) == uint32(lowGUID) })).Return(&mysql.Result{}, nil)

	parser := parser.New()

	proxyInstance := NewTC9Proxy(1, map[uint32]DBConnection{
		1: mockConn1,
	}, parser)

	stmtNode, err := parser.ParseOneStmt("select * from characters where guid = ?", "utf8mb4", "utf8mb4_bin")
	assert.NoError(t, err)

	v := sqlparser.NewCharGUIDFinder()
	stmtNode.Accept(&v)
	v.FillInGUIDIndexes()

	context := &StmtWithParsedDataContext{
		GuidFinder: &v,
		Stmts:      map[uint32]DBStatement{1: mockStmt1},
	}

	proxyInstance.transactionState = TransactionRequested

	_, err = proxyInstance.HandleStmtExecute(context, "", []interface{}{guid.NewCrossrealmPlayerGUID(1, guid.LowType(lowGUID)).GetRawValue()})
	assert.NoError(t, err)

	mockStmt1.AssertExpectations(t)
	mockConn1.AssertExpectations(t)

	assert.Equal(t, uint32(1), proxyInstance.transactionRealmID)
}

func TestHandleStmtExecute_HandlingDummyStmt(t *testing.T) {
	mockConn1 := new(MockDBConnection)
	mockStmt1 := new(MockDBStatement)

	mockConn1.On("Execute", mock.MatchedBy(func(v string) bool { return v == StartTransaction }), mock.Anything).Return(&mysql.Result{}, nil)

	parser := parser.New()

	proxyInstance := NewTC9Proxy(1, map[uint32]DBConnection{
		1: mockConn1,
	}, parser)

	v := sqlparser.NewCharGUIDFinder()

	context := &StmtWithParsedDataContext{
		GuidFinder: &v,
		Stmts:      map[uint32]DBStatement{1: mockStmt1},
		IsDummy:    true,
	}

	proxyInstance.transactionState = TransactionRequested

	_, err := proxyInstance.HandleStmtExecute(context, "", []interface{}{12})
	assert.NoError(t, err)

	mockStmt1.AssertExpectations(t)
	mockConn1.AssertExpectations(t)

	assert.Equal(t, uint32(12), proxyInstance.transactionRealmID)
}
