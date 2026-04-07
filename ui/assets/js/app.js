// If you want to use Phoenix channels, run `mix help phx.gen.channel`
// to get started and then uncomment the line below.
// import "./user_socket.js"

// You can include dependencies in two ways.
//
// The simplest option is to put them in assets/vendor and
// import them using relative paths:
//
//     import "../vendor/some-package.js"
//
// Alternatively, you can `npm install some-package --prefix assets` and import
// them using a path starting with the package name:
//
//     import "some-package"
//
// If you have dependencies that try to import CSS, esbuild will generate a separate `app.css` file.
// To load it, simply add a second `<link>` to your `root.html.heex` file.

// Include phoenix_html to handle method=PUT/DELETE in forms and buttons.
import "phoenix_html"
// Establish Phoenix Socket and LiveView configuration.
import {Socket} from "phoenix"
import {LiveSocket} from "phoenix_live_view"
import {hooks as colocatedHooks} from "phoenix-colocated/council_hub_ui"
import topbar from "../vendor/topbar"

function formatRelativeTime(isoString) {
  const then = new Date(isoString + "Z") // SQLite timestamps are UTC
  const diff = Math.floor((Date.now() - then.getTime()) / 1000)
  if (diff < 5) return "just now"
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return then.toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

const Hooks = {
  CopyMessage: {
    mounted() {
      this.el.addEventListener("click", () => {
        const text = this.el.dataset.copy
        navigator.clipboard.writeText(text).then(() => {
          const icon = this.el.querySelector("span")
          icon.classList.replace("hero-clipboard", "hero-check")
          setTimeout(() => icon.classList.replace("hero-check", "hero-clipboard"), 1500)
        })
      })
    }
  },
  RelativeTime: {
    mounted() {
      this.update()
      this.timer = setInterval(() => this.update(), 30000)
    },
    updated() { this.update() },
    update() {
      const dt = this.el.dataset.timestamp
      if (dt) this.el.textContent = formatRelativeTime(dt)
    },
    destroyed() { clearInterval(this.timer) }
  },
  EmojiPicker: {
    mounted() {
      const trigger = this.el.querySelector(".emoji-picker-trigger")
      const panel = this.el.querySelector(".emoji-picker-panel")
      if (!trigger || !panel) return

      trigger.addEventListener("click", (e) => {
        e.stopPropagation()
        panel.classList.toggle("hidden")
        panel.classList.toggle("flex")
      })

      this._closeHandler = (e) => {
        if (!this.el.contains(e.target)) {
          panel.classList.add("hidden")
          panel.classList.remove("flex")
        }
      }
      document.addEventListener("click", this._closeHandler)
    },
    destroyed() {
      if (this._closeHandler) document.removeEventListener("click", this._closeHandler)
    }
  },
  ScrollBottom: {
    mounted() {
      this.scrollToBottom()
      this.observer = new MutationObserver(() => {
        if (this.isNearBottom()) {
          this.scrollToBottom()
        }
      })
      this.observer.observe(this.el, { childList: true, subtree: true })
    },
    updated() {
      if (this.isNearBottom()) {
        this.scrollToBottom()
      }
    },
    isNearBottom() {
      const threshold = 150
      return this.el.scrollHeight - this.el.scrollTop - this.el.clientHeight < threshold
    },
    scrollToBottom() {
      this.el.scrollTop = this.el.scrollHeight
    },
    destroyed() {
      if (this.observer) this.observer.disconnect()
    }
  }
}

const csrfToken = document.querySelector("meta[name='csrf-token']").getAttribute("content")
const liveSocket = new LiveSocket("/live", Socket, {
  longPollFallbackMs: 2500,
  params: {_csrf_token: csrfToken},
  hooks: {...colocatedHooks, ...Hooks},
})

// Show progress bar on live navigation and form submits
topbar.config({barColors: {0: "#29d"}, shadowColor: "rgba(0, 0, 0, .3)"})
window.addEventListener("phx:page-loading-start", _info => topbar.show(300))
window.addEventListener("phx:page-loading-stop", _info => topbar.hide())

// connect if there are any LiveViews on the page
liveSocket.connect()

// expose liveSocket on window for web console debug logs and latency simulation:
// >> liveSocket.enableDebug()
// >> liveSocket.enableLatencySim(1000)  // enabled for duration of browser session
// >> liveSocket.disableLatencySim()
window.liveSocket = liveSocket

// The lines below enable quality of life phoenix_live_reload
// development features:
//
//     1. stream server logs to the browser console
//     2. click on elements to jump to their definitions in your code editor
//
if (process.env.NODE_ENV === "development") {
  window.addEventListener("phx:live_reload:attached", ({detail: reloader}) => {
    // Enable server log streaming to client.
    // Disable with reloader.disableServerLogs()
    reloader.enableServerLogs()

    // Open configured PLUG_EDITOR at file:line of the clicked element's HEEx component
    //
    //   * click with "c" key pressed to open at caller location
    //   * click with "d" key pressed to open at function component definition location
    let keyDown
    window.addEventListener("keydown", e => keyDown = e.key)
    window.addEventListener("keyup", _e => keyDown = null)
    window.addEventListener("click", e => {
      if(keyDown === "c"){
        e.preventDefault()
        e.stopImmediatePropagation()
        reloader.openEditorAtCaller(e.target)
      } else if(keyDown === "d"){
        e.preventDefault()
        e.stopImmediatePropagation()
        reloader.openEditorAtDef(e.target)
      }
    }, true)

    window.liveReloader = reloader
  })
}

