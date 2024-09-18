local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      --  Let's check if something is wrong with the CRD deployment
      if condition.type == "Ready" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      --  Let's check if things are healthy with the CRD deployment
      if condition.type == "Ready" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end

-- Otherwise let's assume that we are still busy building/deploying the Integration
hs.status = "Progressing"
hs.message = "Waiting for Integration"
return hs
