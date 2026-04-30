local actions = {}
local is_paused = false
if obj.spec.pause == true then
  is_paused = true
end
actions["pause"] = {["disabled"] = is_paused}
actions["unpause"] = {["disabled"] = not is_paused}
return actions
        definitions:
        - name: pause
          action.lua: |
            obj.spec.pause = true
            return obj
        - name: unpause
          action.lua: |
            obj.spec.pause = false
            return obj
