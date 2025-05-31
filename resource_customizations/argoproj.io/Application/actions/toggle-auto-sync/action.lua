function toggleAutoSync(obj)
    if obj.spec.syncPolicy and obj.spec.syncPolicy.automated then
        obj.spec.syncPolicy.automated = nil
        if not next(obj.spec.syncPolicy) then
            obj.spec.syncPolicy = nil
        end
    else
        if not obj.spec.syncPolicy then
            obj.spec.syncPolicy = {}
        end
        obj.spec.syncPolicy.automated = {
            enabled = true
        }
    end
    return obj
end

return toggleAutoSync(obj)