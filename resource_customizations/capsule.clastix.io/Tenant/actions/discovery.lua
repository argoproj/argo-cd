local actions = {}

actions["cordon"] = {
  ["iconClass"] = "fa fa-fw fa-pause",
  ["disabled"] = true,
}
actions["uncordon"] = {
  ["iconClass"] = "fa fa-fw fa-play",
  ["disabled"] = true,
}

local cordoned = false
if obj.spec ~= nil and obj.spec.cordoned ~= nil then
  cordoned = obj.spec.cordoned
end

if cordoned then
  actions["uncordon"]["disabled"] = false
else
  actions["cordon"]["disabled"] = false
end

return actions
