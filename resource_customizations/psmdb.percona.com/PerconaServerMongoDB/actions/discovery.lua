local actions = {}
local is_paused = false
if obj.spec.pause == true then
  is_paused = true
end
actions["pause"] = {["disabled"] = is_paused}
actions["unpause"] = {["disabled"] = not is_paused}
return actions
