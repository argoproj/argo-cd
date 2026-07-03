local actions = {}
actions["create-job"] = {["iconClass"] = "fa fa-fw fa-plus", ["displayName"] = "Create Job"}

if obj.spec.suspend ~= nil and obj.spec.suspend then
    actions["resume"] = {["iconClass"] = "fa fa-fw fa-play" }
else
    actions["suspend"] = {["iconClass"] = "fa fa-fw fa-pause"}
end

return actions
