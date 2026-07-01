-- A bit of documentation about the fields of MedusaTask can be found here:
-- https://docs.k8ssandra.io/reference/crd/k8ssandra-operator-crds-latest/#medusataskstatus
local hs = {}

hs.status = "Unknown"

if obj.status == nil then
	return hs
end

-- We check if we are checking the correct version of obj.status
if obj.status.observedGeneration ~= nil and obj.status.observedGeneration ~= obj.metadata.generation then
	return hs
end

if obj.status.finished == nil and obj.status.failed == nil then
	hs.status = "Progressing"
end

if obj.status.finished ~= nil then
	if obj.status.finished[0] or obj.status.finished[1] then
		hs.status = "Healthy"
	else
		hs.status = "Progressing"
	end
end

if obj.status.failed ~= nil then
	hs.status = "Degraded"
	hs.message = "Failed nodes exist"
end

return hs
