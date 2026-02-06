local hs = {}
if obj.status.pass == true then
    hs.status = "Healthy"
    hs.message = "Analysis is healthy"
    return hs
end
if obj.status.warning == true then
    hs.status = "Healthy"
    hs.message = "Analysis is healthy with warnings"
    return hs
end
if obj.status.pass == false then
    hs.status = "Degraded"
    hs.message = "Analysis is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "Analysis is progressing"
return hs
