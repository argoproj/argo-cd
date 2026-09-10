-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

local hs = {}

hs.status = "Unknown"

if obj.status == nil then
	hs.message = "No status object"
	return hs
end

if obj.status.observedGeneration == nil then
	hs.message = "No observedGeneration"
	return hs
end

-- We check if we are checking the correct version of obj.status
if obj.status.observedGeneration ~= obj.metadata.generation then
	hs.message = "observedGeneration is not equal to generation"
	hs.status = "Progressing"
	return hs
end

if obj.status.finished == nil and obj.status.failed == nil then
	hs.message = "Status not known yet"
	hs.status = "Progressing"
	return hs
end

if obj.status.finished ~= nil then
	if obj.status.finished[0] or obj.status.finished[1] then
		hs.status = "Healthy"
	else
		hs.status = "Progressing"
	end
	return hs
end

if obj.status.failed ~= nil then
	hs.status = "Degraded"
	hs.message = "Failed nodes exist"
end

return hs
