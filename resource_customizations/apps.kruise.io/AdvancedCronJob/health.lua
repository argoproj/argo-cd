hs = { status = "Progressing", message = "AdvancedCronJobs has active jobs" }
-- Extract lastScheduleTime and convert to time objects
lastScheduleTime = nil

if obj.status.lastScheduleTime ~= nil then
  local year, month, day, hour, min, sec = string.match(obj.status.lastScheduleTime, "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z")
  lastScheduleTime = os.time({year=year, month=month, day=day, hour=hour, min=min, sec=sec})
end


if lastScheduleTime == nil and obj.spec.paused == true then 
    hs.status = "Suspended"
    hs.message = "AdvancedCronJob is Paused"
    return hs
end

-- AdvancedCronJobs are progressing if they have any object in the "active" state
if obj.status.active ~= nil and #obj.status.active > 0 then
    hs.status = "Progressing"
    hs.message = "AdvancedCronJobs has active jobs"
    return hs
end
-- AdvancedCronJobs are Degraded if they don't have lastScheduleTime
if lastScheduleTime == nil then
    hs.status = "Degraded"
    hs.message = "AdvancedCronJobs has not run successfully"
    return hs
end
-- AdvancedCronJobs are healthy if they have lastScheduleTime
if lastScheduleTime ~= nil then
    hs.status = "Healthy"
    hs.message = "AdvancedCronJobs has run successfully"
    return hs
end

return hs
