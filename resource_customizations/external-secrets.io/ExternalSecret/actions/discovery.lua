local actions = {}

local disable_refresh = false
local time_units = {"ns", "us", "µs", "ms", "s", "m", "h"}
local digits = obj.spec.refreshInterval
for _, time_unit in ipairs(time_units) do
  digits, _ = digits:gsub(time_unit, "")
  if tonumber(digits) == 0 then
    disable_refresh = true
    break
  end
end

actions["refresh"] = {["disabled"] = disable_refresh}
return actions
