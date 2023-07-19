hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.isFinished then
    if obj.status.ansibleJobResult ~= nil then
      if obj.status.ansibleJobResult.status == "successful" then
        hs.status = "Healthy"
        hs.message = "Done"
      elseif obj.status.ansibleJobResult.status == "error" then
        hs.status = "Degraded"
        hs.message = "Failed"
      end
    end
  end
  if obj.status.ansibleJobResult ~= nil then
    if obj.status.ansibleJobResult.status == "pending" then
      hs.status = "Progressing"
      hs.message = "Job is running"
    end
  end
end
return hs
