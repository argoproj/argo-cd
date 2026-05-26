local hs = {}
hs.status = "Unknown"

if obj.status == nil or obj.status.conditions == nil then
	hs.message = "No conditions found"
	return hs
end

local newestTimestamp = 0
local newestIndex = 0
for idx, condition in pairs(obj.status.conditions) do
	local timestamp = condition.lastTransitionTime
	local format_string = "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z"
	local year, month, day, hour, min, sec =
		string.match(timestamp, format_string)

	local time = os.time({
		day = day,
		month = month,
		year = year,
		hour = hour,
		min = min,
		sec = sec
	})

	if os.difftime(time, newestTimestamp) > 0 then
		newestTimestamp = time
		newestIndex = idx
	end
end

local lastCondition = obj.status.conditions[newestIndex]
if lastCondition.type == 'Healthy' then
	hs.status = "Healthy"
	return hs
end

if lastCondition.status == 'True' then
	hs.status = "Progressing"
end

if lastCondition.status == 'False' then
	hs.status = "Degraded"
end

hs.message = "Last transition type was " .. lastCondition.type

return hs
