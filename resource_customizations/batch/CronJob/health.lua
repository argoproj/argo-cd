hs = {}
-- Extract lastScheduleTime and lastSuccessfulTime and convert to time objects
lastScheduleTime = nil
lastSuccessfulTime = nil
if obj.status.lastScheduleTime ~= nil then
  local year, month, day, hour, min, sec = string.match(obj.status.lastScheduleTime, "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z")
  lastScheduleTime = os.time({year=year, month=month, day=day, hour=hour, min=min, sec=sec})
end
if obj.status.lastSuccessfulTime ~= nil then
  local year, month, day, hour, min, sec = string.match(obj.status.lastSuccessfulTime, "(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)Z")
  lastSuccessfulTime = os.time({year=year, month=month, day=day, hour=hour, min=min, sec=sec})
end
-- CronJobs are progressing if they have any object in the "active" state
if obj.status.active ~= nil and #obj.status.active > 0 then
  hs.status = "Progressing"
  hs.message = "CronJob has active jobs"
  return hs
-- CronJobs are healthy if they don't have lastScheduleTime
elseif lastScheduleTime == nil then
  hs.status = "Healthy"
  hs.message = "CronJob has never run"
  return hs
-- CronJobs are healthy if they have lastScheduleTime and lastSuccessfulTime and lastScheduleTime < lastSuccessfulTime
elseif lastSuccessfulTime ~= nil and lastScheduleTime < lastSuccessfulTime then
  hs.status = "Healthy"
  hs.message = "CronJob has run successfully"
  return hs
else
  hs.status = "Degraded"
  hs.message = "CronJob has not run successfully"
end
return hs
