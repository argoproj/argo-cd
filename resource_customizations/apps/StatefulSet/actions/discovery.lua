local actions = {}
actions["restart"] = {}

actions["scale"] = {
  ["params"] = {
        {
            ["name"] = "scale",
            ["value"] = "",
            ["type"] = "^[0-9]*$",
            ["default"] = tostring(obj.spec.replicas)
        }
  },
  ["hasParameters"] = true, 
  ["errorMessage"] = "Enter any valid number", 
  ["regexp"] = "^[0-9]*$"
}
return actions
