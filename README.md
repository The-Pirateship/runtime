# runtime

**runtime** is a lightweight tool for developers who work develop multi-service apps (frontend, backend, workers etc. all running simultanously)

usually you have to open tabs and run your frontend, backend... one by one.
with runtime, define a `runtime.toml` file, something like this

```runtime.toml
name = "inferenceLake"

[frontend]                  # name of the service
path = "/website"           # path where it is
runCommand = "npm run dev"  # command used to run it

[backend]
path = "/backend"
runCommand = "npm run start"
```

and then from your project root, when you run `runtime dev` or `rt dev` it runs all services using nice zellij tabs.


### installation

> note: runtime uses zellij for the underlying terminal multiplexer, so you'll need that installed first! (`brew install zellij` or details at https://zellij.dev/)

Via homebrew
```
brew tap The-Pirateship/homebrew-runtime
brew install runtime
```

> others coming soon, do open an issue if theres another platform you'd like.

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
```
runtime dev
rt dev // shorthand
```
