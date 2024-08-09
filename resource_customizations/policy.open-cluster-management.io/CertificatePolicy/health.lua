hs = {}
if obj.status == nil or obj.status.compliant == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.compliant == "Compliant" then
  hs.status = "Healthy"
  hs.message = "All certificates found comply with the policy"
  return hs
else
  hs.status = "Degraded"
  hs.message = "At least once certificate does not comply with the policy"
  return hs
end
