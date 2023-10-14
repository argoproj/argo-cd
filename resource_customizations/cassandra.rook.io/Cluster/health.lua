local hs = {}
if obj.status ~= nil then
  if obj.status.racks ~= nil then
    local all_racks_good = true
    for key, value in pairs(obj.status.racks) do
      if all_racks_good and value.members ~= nil and value.readyMembers ~= nil and value.members ~= value.readyMembers then
        all_racks_good = false
        break
      end
    end
    if all_racks_good then
      hs.status = "Healthy"
    else
      hs.status = "Progressing"
      hs.message = "Waiting for Cassandra Cluster"
    end
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Cassandra Cluster"
return hs

