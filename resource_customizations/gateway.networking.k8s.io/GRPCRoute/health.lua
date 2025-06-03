-- api info here: https://gateway-api.sigs.k8s.io/reference/spec/#grpcroute

hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status ~= nil and obj.status.parents ~= nil and obj.status.parents.conditions ~=nil then
    if obj.status.parents.conditions.type == "Accepted" and obj.status.parents.conditions.status == "True" then
        hs.status = "Healthy"
        hs.message = obj.status.parents.conditions.message
    elseif obj.status.parents.conditions.type == "Accepted" and obj.status.parents.conditions.status == "False" then
        hs.status = "Degraded"
        hs.message = obj.status.parents.conditions.message
    end
end