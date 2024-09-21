hs = {}
if obj.status ~= nil and obj.status.state ~= nil then
    if obj.status.state == "CREATED" and obj.status.connectorState == "RUNNING" then
        hs.status = "Healthy"
        hs.message = "Connector running"
        return hs
    end
    if obj.status.state == "ERROR" then
        hs.status = "Degraded"
        if obj.status.conditions and #obj.status.conditions > 0 then
            hs.message = obj.status.conditions[1].message -- Kafka Connector only has one condition and nests the issues in the error message here
        else
            hs.message = "No conditions available"
        end
        return hs
    end
end
hs.status = "Progressing"
hs.message = "Waiting for Kafka Connector"
return hs