# D2. Models are one package per driver under `models/<driver>`

Status: Amended by D71.

Each driver gets one package, named after the driver, under `models/`. One
package covers every supported version of that database, because D8 holds the
version differences as data inside it.

D71 amends the part that said a generator produces the code. Nothing does. The
package layout this decision chose is what stands.
