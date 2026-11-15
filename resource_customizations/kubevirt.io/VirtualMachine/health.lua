local hs = { status="Progressing", message="No status available"}

-- A VirtualMachine whose desired state is "stopped" is Healthy once it has reached that state.
-- KubeVirt maps spec.running: false onto runStrategy: Halted, and defines Halted as "the system
-- is asked to ensure that no VM is running", so the two spell the same desired state. The other
-- run strategies (Always, RerunOnFailure, Once, Manual) can each leave the machine stopped
-- without the manifest having asked for it, so they keep the status they had.
-- status.created reports whether a VirtualMachineInstance exists, so a machine that is declared
-- stopped while one still exists is still shutting down and is not healthy yet.
local declaredStopped = false
if obj.spec ~= nil then
  if obj.spec.running == false or obj.spec.runStrategy == "Halted" then
    declaredStopped = true
  end
end
if declaredStopped and (obj.status == nil or not obj.status.created) then
  hs.status = "Healthy"
  hs.message = "Stopped as declared"
  return hs
end

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
