# Importing a Plugin is an upload of code

A Theme is one file that cannot do anything, so an endpoint that writes it costs disk and nothing else. A Plugin is a different animal. Its folder holds widget modules the Dashboard imports into the page, and optionally a `backend` command that core starts as a subprocess **with the panel's own privileges** ([ADR-0003](0003-plugin-subprocess-vanilla-esm.md)). An endpoint that takes a Plugin is an endpoint that accepts a program to run.

That is why the import waited for a decision rather than shipping with the rest of the Plugin work. The options were to hold it until there is a login, or to ship it as what it is: a trusted-network feature, with the reach it grants said out loud.

**We ship it, on ADR-0001's terms.** Star Panel is self-hosted behind the owner's own access control and has no user model by design ([ADR-0001](0001-single-user-no-auth.md)), so this endpoint does not invent one. The honest way to put it: it widens who can install a Plugin from *whoever can write to the server's disk* to *whoever can reach the port*. On a trusted network that is the same person; on an untrusted one neither was ever safe.

What follows from the decision:

- **The panel says so.** The Plugins section in edit mode states that importing runs code and that there is no login, next to the file picker — the reading a person gets before they pick a file, not after.
- **The panel still never installs anything.** It reports a Plugin's unmet `requires` and leaves installing the runtime to the owner ([docs/PLUGINS.md](../PLUGINS.md)).
- **An upload is unpacked, never overwritten.** A name already in `plugins/` is refused and the message names the folder. Only the archive's own folder is written, as plain files: an entry that climbs out of it is refused, and a symlink is not unpacked at all.
- **If a login ever arrives**, this endpoint is the first thing that should carry it, and the warning in the UI becomes unnecessary.
