actions = {}
actions["terminate"] = {["disabled"] = (obj.spec.terminate or
    obj.status.phase == "Successful" or
    obj.status.phase == "Failed" or
    obj.status.phase == "Error" or
    obj.status.phase == "Inconclusive"
)}
return actions
