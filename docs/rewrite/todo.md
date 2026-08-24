# TODO Rewrite

 - eqldb: send items / kills
 - Overlay: Combat timeout / Remove last fight

## Not in v1
 - DPS: Combat History row limit?
 - DPS: Min/Max values
 - Parser: Replay error management
 - Parser: Handle truncated files
 - Parser: ?Logrotate

 ## Done
 - ~~Store "Last Logfiles" and make them selectable~~
 - ~~Event: Spell Icon extraction~~
 - ~~Event: Spell Icon settings~~
 - ~~XPHour: XP in this level~~
 - ~~XPHour: Time to levelup~~
 - ~~Event: RegExp validation~~
 - ~~eqldb: authorization workflow~~
 - ~~eqldb: identify inventory export~~
 - ~~eqldb: upload inventory~~
 - ~~eqldb: send sky drops to eqldb~~
 - ~~Overlay: Scaling~~
 - ~~DPS: Filter~~
 - ~~DPS: track engagement for SDPS~~
 - ~~Sky: Open Quest reward in browser~~
 - ~~Sky: Watch Quests~~
 - ~~Overlay: Opacity~~
 - ~~Overlay: Position/Size persistend~~
 - ~~Mainwindow: Position/Size persistence~~
 - ~~Windows only "always on top"~~


 Fight timeout:

- Live timeout is disabled. The ticker branch in [module.go](/home/jk/projects/go/eqdps/internal/module/dps/module.go:107) contains only commented code. Current fights therefore never time out while following a live log.
- Current timeout is hardcoded to 20 seconds in [combat.go](/home/jk/projects/go/eqdps/internal/module/dps/combat.go:61). Legacy uses a configurable 5–60 seconds, defaulting to 15.
- Current timeout measures from `Fight.End`, but [fight.go](/home/jk/projects/go/eqdps/internal/data/fight.go:104) deliberately does not advance `End` for DoT or damage-shield events. Legacy live timeout advances activity for every damage event.
- Legacy removes a completed fight from the overlay after the configured timeout. The current overlay keeps the last published fight indefinitely.

Overlay fight selection:

- Current selection considers only active fights while any exist and chooses the greatest `LastParticipate` timestamp in [module.go](/home/jk/projects/go/eqdps/internal/module/dps/module.go:121).
- Legacy considers the last intentionally attacked fight across both active and recently completed fights. Thus, an incidental active fight does not replace the player’s last intentional target.
- Legacy uses a monotonically increasing order number. Current uses logfile timestamps, so two targets intentionally damaged during the same second cannot be reliably ordered.
- Legacy waits 200 ms before changing overlay focus, allowing rapid target changes to settle. Current switches immediately.
- If no fight has intentional participation, legacy chooses the newest active fight. Current fights all have zero `LastParticipate`, so selection effectively falls back to their creation order.
- When nothing is active, current chooses the most recently created fight, not necessarily the fight most recently completed or intentionally attacked.

Intentional-participation classification:

- Legacy excludes `cleave`, `kick`, `punch`, `reave`, `strike`, their plural forms, and ripostes. Current excludes only `Cleave`, `Kick`, and ripostes in [combat.go](/home/jk/projects/go/eqdps/internal/module/dps/combat.go:311).
- Current accepts every matching spell hit within ten seconds of a cast. Legacy accepts the first matching timestamp and additional hits only at that exact timestamp. This can cause the current overlay to treat later proc-like hits as intentional.

Damage-to-fight assignment:

- The normal source/target mob selection in `getActiveFightLegacy` mostly matches the legacy `mobForEvent`.
- After death, legacy buffers ambiguous DoTs to distinguish a new same-named mob. Current immediately adds them to the grace-period fight.
- Current also retains damage-shield events in the grace-period fight; legacy retention applies only to DoTs. This is the intentional behavior you chose earlier.
