module("luci.controller.cloudflare-ddns", package.seeall)

function index()
	entry({"admin", "services", "cloudflare-ddns"},
	      cbi("cloudflare-ddns"),
	      _("Cloudflare DDNS"), 60)
end
