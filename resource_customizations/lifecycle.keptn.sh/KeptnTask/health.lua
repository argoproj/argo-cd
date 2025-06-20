local hs = {}
if obj.status.status == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnTask is healthy"
    return hs
end
if obj.status.status == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnTask is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnTask is progressing"
return hs