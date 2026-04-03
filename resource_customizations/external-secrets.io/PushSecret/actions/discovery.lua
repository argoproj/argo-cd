local actions = {}

local disable_push = false
local time_units = {"ns", "us", "µs", "ms", "s", "m", "h"}
local digits = obj.spec.refreshInterval
if digits ~= nil then
  digits = tostring(digits)
  for _, time_unit in ipairs(time_units) do
    if digits == "0" or digits == "0" .. time_unit then
      disable_push = true
      break
    end
  end
end

actions["push"] = {["disabled"] = disable_push}
return actions
