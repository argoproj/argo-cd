local hs = {}

local amountOfNodes = 0
if obj.status ~= nil and obj.status.nodeStatuses ~= nil then
	for _, _ in pairs(obj.status.nodeStatuses) do
		amountOfNodes = amountOfNodes + 1
	end
else
	hs.status = "Degraded"
	hs.message = "No nodeStatus in object's status"
	return hs
end

if obj.spec ~= nil and obj.spec.size ~= nil then
	if obj.spec.size == amountOfNodes then
		hs.status = "Healthy"
		return hs
	else
		hs.status = "Degraded"
		hs.message = "Size in spec and number of nodes in status differ"
	end
else
	hs.status = "Degraded"
	hs.message = "No size in object's spec"
	return hs
end

return hs
