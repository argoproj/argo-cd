local actions = {}
actions["restart"] = {}
actions["scale"] = {["defaultValue"] = tostring(obj.spec.replicas), ["hasParameters"] = true, ["errorMessage"] = "Enter any valid number", ["regexp"] = "^[0-9]*$"}
return actions
