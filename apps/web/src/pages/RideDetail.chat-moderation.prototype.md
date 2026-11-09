# Chat moderation menu prototype

Question: how can ride-room messages keep report and rider-safety controls
available without repeating four links beneath every message?

The owner selected the persistent message ellipsis menu on 9 November 2026
after comparing message menus, author menus and a bottom sheet. Implemented
through [issue 25](https://github.com/unstppbl/bikers/issues/25) and
[the implementation pull request](https://github.com/unstppbl/bikers/pull/27).

This branch preserves the complete interactive comparison as a primary source.
It is a throwaway prototype and must not be merged into main. The main branch
keeps only the implemented decision.

The HTML is the original inline fragment. Its actions are simulated and its
administrator toggle is a preview. It makes no moderation or network calls.
Static JavaScript and DOM checks covered variants, report scope, role
visibility, keyboard/focus/closing behavior and host state APIs returning void
or Promise. Browser visual acceptance was unavailable because local browser
use was disabled.

Original HTML SHA256: 68ef3558bafd1bda6914cbc6b1f90d5be9a59f566e9c7254080430b6700519cc.
Implementation parent: adbf86e9f04382b0c857c11bd0dc89caa4198c2a.
