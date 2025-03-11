local actions = {}

local push_disabled = false
if obj.spec.refreshInterval == "0s" then
  push_disabled = true
end

actions["push"] = {["disabled"] = push_disabled}
return actions
