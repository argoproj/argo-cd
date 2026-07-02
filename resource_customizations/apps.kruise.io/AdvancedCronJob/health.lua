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
