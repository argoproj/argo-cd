actions = {}
len = 1

if obj.spec.paused ~= nil and obj.spec.paused then
    pause = {}
    pause["name"] = "resume"
    actions[len]=pause
    len = len + 1
end

return actions
