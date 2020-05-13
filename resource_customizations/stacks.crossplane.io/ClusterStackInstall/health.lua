hs = {
  status = "Progressing",
  message = "Waiting for stack to be installed"
}
if obj.status ~= nil then
  if obj.status.conditionedStatus ~= nil then
    if obj.status.conditionedStatus.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditionedStatus.conditions) do
        if condition.type == "Ready" then
          hs.message = condition.reason
          if condition.status == "True" then
            hs.status = "Healthy"
            return hs
          end
        end
      end
    end
  end
end
return hs
