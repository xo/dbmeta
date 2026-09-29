# D129. An embedded product waits for a model

Status: Amends D66.

Ken decided on 2026-09-29 that chai, csvq and ql wait for a model, as every
other Staged release does, and stay in dbrun.

D66 said that a product whose only local option is an emulator is Archived on
arrival, and that chai, csvq and ql get no model at all. D119 made all of them
Staged, and a Staged release is one that waits for a model, so the two
decisions disagreed. This settles it in D119's favor. chai, csvq and ql are
Staged with the cadence they record (D120), and the emulators of hosted
services, such as Spanner's, are Staged the same way, which is what their
entries already say. D66's order still decides which comes when.
