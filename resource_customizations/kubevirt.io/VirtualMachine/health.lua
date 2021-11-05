hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Paused" and condition.status == "True" then
        hs.status = "Suspended"
        hs.message = "Paused"
        return hs
      end
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status="Healthy"
          hs.message="Running"
        else
          if obj.status.created then
	    hs.message = "Starting"
          else
            hs.status = "Suspended"
            hs.message = "Stopped"
          end
        end
      end
    end
  end
  if obj.status.printableStatus ~= nil then
    hs.message = obj.status.printableStatus
  end
end
return hs
