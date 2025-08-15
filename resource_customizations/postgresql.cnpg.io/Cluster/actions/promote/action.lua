local os = require("os")
local nextIndex = 0
for index, node in pairs(obj.status.instancesStatus.healthy) do
    if node == obj.status.currentPrimary then
        nextIndex = index + 1
        if nextIndex > #obj.status.instancesStatus.healthy then
            nextIndex = 1
        end
        break
    end
end
if nextIndex > 0 then
    obj.status.targetPrimary = obj.status.instancesStatus.healthy[nextIndex]
    obj.status.targetPrimaryTimestamp = os.date("!%Y-%m-%dT%XZ")
end
return obj
