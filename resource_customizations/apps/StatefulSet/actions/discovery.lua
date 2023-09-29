local actions = {}
actions["restart"] = {}
actions["scale"] = {["defaultValue"] = tostring(obj.spec.replicas), ["hasParameters"] = true}
return actions
