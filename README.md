# runtime

many projects have a frontend, backend and some other services that you have to run together. Runtime is a lightweight utility to specificy services in a config file and then run them all at once in a multiplexer.


### installation

> note: runtime uses zellij for the underlying terminal multiplexer, so you'll need that installed first! (`brew install zellij`)

Via homebrew
```
brew tap The-Pirateship/homebrew-runtime
brew install runtime
```

### Steps

#### 1) Create a `runtime.toml` file in your project root

```runtime.toml
name = "inferenceLake"

[frontend]                  # name of the service
path = "/website"           # path where it is
runCommand = "npm run dev"  # command used to run it

[backend]
path = "/backend"
runCommand = "npm run start"

[llm-proxy]
path = "/workers/llm-proxy"
runCommand = "bunx wrangler dev"
```

#### 2) Run your project!

now from your project root, you can run
```runtime dev```

or for short
```rt dev```
