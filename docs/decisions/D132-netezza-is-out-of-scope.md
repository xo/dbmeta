# D132. Netezza is out of scope

Status: Decided.

Ken decided on 2026-09-29 that Netezza is out of scope and is removed. As far
as he can tell it was never tested, nobody uses it, and there is no copy of it
to test with.

IBM publishes no container image of Netezza. Its only local option is the
Netezza emulator, a virtual machine image that IBM gives to members of the
Netezza Developer Network, as is and unsupported, and that runs a virtual
machine of its own inside it. A person has to join the network to download
it, and an agent does not sign up for anything (D117, D118).

So dbmeta has no Netezza model and no entry, and usql's Netezza driver,
which changed a password with no escaping and read metadata through the
shared information_schema reader, has nothing here to move to. Db2 was
decided the same way, for a different reason (D130).
