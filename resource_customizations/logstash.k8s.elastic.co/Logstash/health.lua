local hs = {}
if obj.status ~= nil and obj.status.health ~= nil then
  if obj.status.health == "green" then
    hs.status = "Healthy"
    hs.message = "Logstash status is Green"
    return hs
  elseif obj.status.health == "yellow" then
    hs.status = "Progressing"
    hs.message = "Logstash status is Yellow"
    return hs
  elseif obj.status.health == "red" then
    hs.status = "Degraded"
    hs.message = "Logstash status is Red"
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Logstash"
return hs
