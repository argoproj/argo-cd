hs = { status = "Healthy", message = "Protection policy not yet executed" }
if obj.status ~= nil then
  if obj.status.lastScheduleTime ~= nil then
    hs.message = "Protection policy lastScheduleTime: " .. obj.status.lastScheduleTime
  end
end
return hs
