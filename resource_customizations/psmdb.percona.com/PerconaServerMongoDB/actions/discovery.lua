local actions = {}
local is_paused = false
if obj.spec.pause == true then
  is_paused = true
end
actions["pause"] = {
    ["disabled"] = is_paused,
    ["iconClass"] = "fa fa-fw fa-pause-circle"
}
actions["unpause"] = {
    ["disabled"] = not is_paused,
    ["iconClass"] = "fa fa-fw fa-play-circle"
}
return actions
