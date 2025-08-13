local actions = {}

local disable_push = false
local time_units = {"ns", "us", "µs", "ms", "s", "m", "h"}
local digits = obj.spec.refreshInterval
for _, time_unit in ipairs(time_units) do
  digits, _ = digits:gsub(time_unit, "")
  if tonumber(digits) == 0 then
    disable_push = true
    break
  end
end

actions["push"] = {["disabled"] = disable_push}
return actions
