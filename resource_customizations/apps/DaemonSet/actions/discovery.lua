local actions = {}
actions["restart"] = {}
actions["scale"] = {["defaultValue"] = tostring(obj.spec.replicas), ["hasParameters"] = true,  ["errorMessage"] = "Enter any valid number more than 0", ["regexp"]= "^[1-9][0-9]*$"}
return actions
