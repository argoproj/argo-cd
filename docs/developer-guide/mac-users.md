# MacOS users
Below are known issues specific to macOS

## Firewall dialogs
If you get firewall dialogs, you can click "Deny", since no access from outside your computer is typically desired.

## Exec format error for virtualized toolchain make targets
If you get `/go/src/github.com/argoproj/argo-cd/dist/mockery: cannot execute binary file: Exec format error`, this typically means that you ran a virtualized `make` target after you ran the a local `make` target.   
To fix this and continue with the virtualized toolchain, delete the contents of `argo-cd/dist` folder.   
If later on you wish to run `make` targets of the local toolchain again, run `make install-tools-local` to re-populate the contents of the `argo-cd/dist` folder.
