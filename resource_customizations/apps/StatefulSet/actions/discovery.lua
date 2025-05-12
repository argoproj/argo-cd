local actions = {}
actions["restart"] = {}

actions["scale"] = {
  ["params"] = {
        {
            ["name"] = "replicas",
            ["default"] = tostring(obj.spec.replicas)
        }
  },
}
return actions
