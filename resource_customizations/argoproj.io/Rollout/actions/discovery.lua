actions = {}
actions["restart"] = {["disabled"] = false}

local paused = false
if obj.status ~= nil and obj.status.pauseConditions ~= nil then
    paused = table.getn(obj.status.pauseConditions) > 0
elseif obj.spec.paused ~= nil then
    paused = obj.spec.paused
end
actions["resume"] = {["disabled"] = not(paused)}

fullyPromoted = obj.status.currentPodHash == obj.status.stableRS
actions["abort"] = {["disabled"] = fullyPromoted or obj.status.abort}
actions["retry"] = {["disabled"] = fullyPromoted or not(obj.status.abort)}

actions["promote-full"] = {["disabled"] = true}
if obj.status ~= nil and not(fullyPromoted) then
    generation = tonumber(obj.status.observedGeneration)
    if generation == nil or generation > obj.metadata.generation then
        -- rollouts v0.9 - full promotion only supported for canary
        actions["promote-full"] = {["disabled"] = obj.spec.strategy.blueGreen ~= nil}
    else
        -- rollouts v0.10+
        actions["promote-full"]["disabled"] = false
    end
end

return actions
