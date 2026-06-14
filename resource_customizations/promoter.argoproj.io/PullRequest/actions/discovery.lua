local actions = {}

actions["merge"] = {
    ["iconClass"] = "fa fa-fw fa-code-merge",
    ["disabled"] = obj.spec.state ~= "open",
    ["displayName"] = "Merge PR"
}
return actions
