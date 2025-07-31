local actions = {}
actions["restart"] = {}

actions["scale"] = {
  ["params"] = {
        {
            ["name"] = "replicas"
        }
  },
}
return actions
