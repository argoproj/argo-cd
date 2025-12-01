local actions = {}
 
actions["restart"] = {
    ["iconClass"] = "fa fa-fw fa-redo"
}
 
actions["scale"] = {
    ["iconClass"] = "fa fa-fw fa-plus-circle",
    ["params"] = {
        {
            ["name"] = "replicas"
        }
    },
}
 
return actions
