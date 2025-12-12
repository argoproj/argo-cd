local actions = {}

local disable_refresh = false
local time_units = {"ns", "us", "µs", "ms", "s", "m", "h"}
local digits = obj.spec.refreshInterval
local policy = obj.spec.refreshPolicy
if digits ~= nil then
  digits = tostring(digits)
  for _, time_unit in ipairs(time_units) do
    if (digits == "0" or digits == "0" .. time_unit) and policy ~= "OnChange" then
      disable_refresh = true
      break
    end
  end
end

actions["refresh"] = {["disabled"] = disable_refresh}
return actions
