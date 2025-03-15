local actions = {}

local refresh_disabled = false
if obj.spec.refreshInterval == "0s" then
  refresh_disabled = true
end

actions["refresh"] = {["disabled"] = refresh_disabled}
return actions
