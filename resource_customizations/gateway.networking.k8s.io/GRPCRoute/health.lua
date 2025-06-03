-- api info here: https://gateway-api.sigs.k8s.io/reference/spec/#grpcroute

hs = {
  status = "Progressing",
  message = "Waiting for status",
}

if obj.status and obj.status.parents then
    for _, parent in ipairs(obj.status.parents) do
        if parent.conditions then
            for _, cond in ipairs(parent.conditions) do
                -- print("Condition type:", cond.type, "status:", cond.status, "message:", cond.message)
                if cond.type == "Accepted" and cond.status == "True" then
                    hs.status = "Healthy"
                    hs.message = cond.message
                    return hs
                elseif cond.type == "Accepted" and cond.status == "False" then
                    hs.status = "Degraded"
                    hs.message = cond.message
                    return hs
                end
            end
        end
    end
end

return hs