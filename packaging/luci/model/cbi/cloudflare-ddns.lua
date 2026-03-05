local pid = (luci.sys.exec("pidof scfddns 2>/dev/null") or ""):gsub("%s+", "")
local refresh_url = luci.dispatcher.build_url("admin", "services", "cloudflare-ddns", "config")
local refresh_btn = "&nbsp;&nbsp;<a class='btn cbi-button' href='" .. refresh_url .. "' style='padding:2px 10px;font-size:0.85em'>刷新</a>"
local status_html
if pid ~= "" then
	status_html = "<span style='color:#4caf50;font-weight:bold'>&#9679; 运行中</span>&nbsp;&nbsp;PID: " .. pid .. refresh_btn
else
	status_html = "<span style='color:#f44336;font-weight:bold'>&#9679; 未运行</span>" .. refresh_btn
end

m = Map("cloudflare-ddns",
	translate("Cloudflare DDNS"),
	translate("自动检测公网 IP 变化并更新 Cloudflare DNS 记录。"))

m.on_after_commit = function(self)
	local enabled = self.uci:get("cloudflare-ddns", "main", "enabled")
	if enabled == "1" then
		os.execute(": > /var/log/cloudflare-ddns.log 2>/dev/null")
		os.execute("/etc/init.d/cloudflare-ddns enable >/dev/null 2>&1")
		os.execute("/usr/sbin/cloudflare-ddns-ctl stop >/dev/null 2>&1")
		os.execute("/usr/sbin/cloudflare-ddns-ctl start >/dev/null 2>&1")
	else
		os.execute("/usr/sbin/cloudflare-ddns-ctl stop >/dev/null 2>&1")
		os.execute("/etc/init.d/cloudflare-ddns disable >/dev/null 2>&1")
	end
end

s = m:section(NamedSection, "main", "cloudflare-ddns")
s.addremove = false
s.anonymous = false

-- 服务状态（放在启用开关上方）
o = s:option(DummyValue, "_status", translate("服务状态"))
o.rawhtml = true
function o.cfgvalue() return status_html end

o = s:option(Flag, "enabled", translate("启用"))
o.rmempty  = false
o.default  = "0"

o = s:option(Value, "api_token", translate("API Token"),
	translate("在 Cloudflare 控制台 → 「我的个人资料」→「API 令牌」→「创建令牌」中生成，需要 Zone:DNS:Edit 权限。"))
o.password = true
o.rmempty  = false

o = s:option(Value, "zone_id", translate("Zone ID"),
	translate("在 Cloudflare 控制台进入对应域名，右侧「API」区域可直接复制 Zone ID。"))
o.rmempty = false

o = s:option(Value, "record_name", translate("记录名称"),
	translate("要更新的 DNS 记录全名，例如 home.example.com"))
o.rmempty = false

o = s:option(ListValue, "record_type", translate("记录类型"))
o:value("A",    "A（IPv4）")
o:value("AAAA", "AAAA（IPv6）")
o.default = "A"

o = s:option(Value, "check_interval", translate("检查间隔（秒）"))
o.datatype = "uinteger"
o.default  = "300"

o = s:option(Value, "ip_urls", translate("IP 检测服务地址"),
	translate("逗号分隔的 URL 列表，留空则根据记录类型自动选择内置默认地址（icanhazip / ifconfig.co / ipify）。"))
o.rmempty  = true

return m
