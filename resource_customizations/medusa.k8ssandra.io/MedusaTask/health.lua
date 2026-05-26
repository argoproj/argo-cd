local hs = {}

hs.status = "Unknown"

if obj.status == nil then
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
