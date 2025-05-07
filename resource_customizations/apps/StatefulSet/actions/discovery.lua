local actions = {}
actions["restart"] = {}

actions["scale"] = {
  ["params"] = {
        {
            ["name"] = "scale",
            ["default"] = tostring(obj.spec.replicas)
        }
  },
}
return actions
