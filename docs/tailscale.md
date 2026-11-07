# Put Council Hub on your tailnet

Council Hub can join your Tailscale network as its own device, so your other machines reach it at `https://<name>.<tailnet>.ts.net/mcp` with no port forwarding and no extra container. This is separate from [clustering over Tailscale](clustering-tailscale.md), which links Council Hub nodes to each other.

## Turn it on

1. Open **Settings** in the UI and find the **Tailscale** card.
2. Click **Sign in with Tailscale**. Your browser opens the Tailscale sign-in page. If the browser blocks the pop-up, use **Continue in your browser** or **Copy link** on the card.
3. Approve the device. The card shows **starting** while HTTPS is set up, then **running** with the URL and a ready-to-copy `claude mcp add` command.

**Cancel sign-in** stops a sign-in you do not want. **Sign out** signs the device out of Tailscale and turns the feature off; the next enable needs a new sign-in.

Your tailnet needs **MagicDNS** and **HTTPS certificates** turned on (admin console, DNS). Without HTTPS certificates the card shows an error that says so.

## Who can connect

Every request on the tailnet is checked against the caller's Tailscale identity. The default is **owner only**: the Tailscale user who signed this device in. **Members** (under *Advanced*) admits any user on the tailnet. A tagged device has no owner, so tags need **members** mode. Anything not admitted gets 403.

The tailnet side serves only `/mcp` and `/health`. The Settings API (`/api/ui/*`) and the cluster API (`/api/internal/*`) are not reachable over the tailnet.

## Settings

| Setting | Default | Notes |
|---|---|---|
| Device name | `council-hub` | Becomes the first part of the URL |
| Who can connect | owner | `owner` or `members` |
| Tags | none | For example `tag:council`; needs `members` |
| Control server | Tailscale's | Set a URL to use Headscale |
| Auth key | none | For headless sign-in; used once, never saved |

The choices are saved to `tailscale/config.json` next to the database, and the device's keys to `tailscale/state`. In the Docker image both are under `/data`, so the `/data` volume keeps the device across restarts: it rejoins without a new sign-in.

## Notes

- **Sign out leaves the device in your list.** Tailscale keeps a signed-out device in the admin console, offline, until you delete it there.
- **Clustering is unchanged.** Erlang cluster traffic belongs to the UI process, so nodes still link through the sidecar setup in [clustering-tailscale.md](clustering-tailscale.md).
