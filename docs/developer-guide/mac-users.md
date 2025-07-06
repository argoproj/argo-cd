# MacOS users
Below are known issues specific to macOS

## Port 5000

You may get an error listening on port 5000 on macOS:

```text
docker: Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:5000 -> 0.0.0.0:0: listen tcp 0.0.0.0:5000: bind: address already in use.
```

In that case, you can disable "AirPlay Receiver" in macOS System Preferences.

## Firewall dialogs
If you get firewall dialogs, you can click "Deny", since no access from outside your computer is typically desired.
