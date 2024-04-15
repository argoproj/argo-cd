hs = {}
if obj.status ~= nil then
  if obj.status.ansibleJobResult ~= nil then
    jobstatus = obj.status.ansibleJobResult.status
    if jobstatus == "successful" then
      hs.status = "Healthy"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
    if jobstatus == "failed" or jobstatus == "error" or jobstatus == "canceled" then
      hs.status = "Degraded"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
    if jobstatus == "new" or jobstatus == "pending" or jobstatus == "waiting" or jobstatus == "running" then
      hs.status = "Progressing"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for AnsibleJob"
return hs
