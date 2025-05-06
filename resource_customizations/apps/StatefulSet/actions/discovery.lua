local actions = {}
actions["restart"] = {}

actions["scale"] = {
  ["params"] = {
        {
            ["name"] = "scale",
            ["value"] = "",
            ["default"] = tostring(obj.spec.replicas)
        }
  },
}
return actions
