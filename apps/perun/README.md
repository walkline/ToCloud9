# Perun

Perun, named after the Slavic god, is a tool for managing multiple applications. 
It provides features like running several apps simultaneously, auto-restarting apps upon failure, logging app output to files, and live viewing of logs from all apps.

![](.github/images/perun.png "Perun console")

## Configuration

Perun uses a YAML configuration file to define the apps it manages. 
Below is an example configuration file:

```
perun:
  apps:
    # Configuration for the servers-registry app
    - name: "servers-registry"
      # Alias is used as a reference to this app.
      # If you set an alias like "aa", then you can use it in commands; for example, "restart aa" will restart this app.
      alias:
        - "sr"
      # Binary is the path to the binary/executable file.
      binary: "./servers-registry"
      # Args are the app argument variables that will be passed on launch.
      args: ""
      # Env is the environment variables to pass to the app.
      env: {}
      # WorkingDir is the working dir that will be passed to the app.
      workingDir: ""
      # StartupTimeoutSecs is the timeout time for starting the app in seconds.
      startupTimeoutSecs: 10
      # PartOfAppStartedLogMsg is used to determine if the app started.
      # Example: If your app prints a log like this when the app started: "MySupper app successfully started.",
      # then you can set this variable to "successfully started" or "started".
      partOfAppStartedLogMsg: "successfully started"
    # Configuration for the guidserver app
    - name: "guidserver"
      alias:
        - "guid"
      binary: "./guidserver"
      args: ""
      env: {}
      workingDir: ""
      startupTimeoutSecs: 10
      partOfAppStartedLogMsg: "successfully started"
    # Configuration for the authserver app
    - name: "authserver"
      alias:
        - "as"
        - "auth"
      binary: "./authserver"
      args: ""
      env: {}
      workingDir: ""
      startupTimeoutSecs: 10
      partOfAppStartedLogMsg: "successfully started"
    # Add more apps as needed
```

## Console Commands

Perun's console supports the following commands:

* `restart $appname1 $appname2 ...`: Restarts the specified applications.
  * Aliases: `r`, `re`, `start`
* `stop $appname1 $appname2 ...`: Stops the specified apps.
* `clear`: Clears the logs output.
  * Alias: `cl` 
* `focus $appname1 $appname2 ...`: Displays logs only for the specified apps. You can pass "focus all" to show output for all apps.
  * Aliases: `f`, `fo`

## Usage
1. Prepare your config.yml file as shown above.
2. Place config file in the same dir as perun binary, or specify config file with `perun -c path/to/config.yml`.
3. Run perun. 


