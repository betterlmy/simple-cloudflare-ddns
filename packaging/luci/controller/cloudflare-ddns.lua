module("luci.controller.cloudflare-ddns", package.seeall)

function index()
	entry({"admin", "services", "cloudflare-ddns"},
	      firstchild(), _("Cloudflare DDNS"), 60).dependent = false
	entry({"admin", "services", "cloudflare-ddns", "config"},
	      cbi("cloudflare-ddns"), _("配置"), 1)
	entry({"admin", "services", "cloudflare-ddns", "log"},
	      call("action_log"), _("日志"), 2)
	entry({"admin", "services", "cloudflare-ddns", "log_clear"},
	      call("action_log_clear")).leaf = true
end

function action_log()
	local log = luci.sys.exec("tail -n 200 /var/log/cloudflare-ddns.log 2>/dev/null") or ""
	luci.template.render("cloudflare-ddns/log", {log = log})
end

function action_log_clear()
	luci.sys.exec("echo -n > /var/log/cloudflare-ddns.log 2>/dev/null")
	luci.http.redirect(luci.dispatcher.build_url("admin", "services", "cloudflare-ddns", "log"))
end
