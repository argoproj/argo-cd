local hs = {}

local cnpgStatus = {
  ["started"] = "Progressing",
  ["running"] = "Progressing",
  ["running"] = "Progressing",
  ["finalizing"] = "Progressing",
  ["completed"] = "Healthy",
  ["failed"] = "Degraded",
  ["walArchivingFailing"] = "Degraded",
}

if obj.status ~= nil and obj.status.phase ~= nil then
  statusState = cnpgStatus[obj.status.phase]
  if statusState ~= nil then
    hs.status = statusState
    hs.message = "Backup " .. obj.status.phase
    return hs
  else
    hs.status = "Unknown"
    hs.message = "Backup " .. obj.status.phase
    return hs
  end
end

hs.status = "Unknown"
hs.message = "Backup status unknown"
return hs
