# LAN Mode

LAN mode is for playing around one shared screen, with local players using
separate physical keyboards for guessing and a separate drawing device for the
current drawer. It is meant for a party setup: one computer hosts the lobby and
shows the main game, other devices on the same network open the drawing or
guessing screens.

## What You Need

- One computer running Scribble.rs and showing the main lobby.
- One keyboard per local player, plugged into the host computer.
- Optional but recommended: a tablet, phone, or laptop for the drawing screen.
- All devices on the same Wi-Fi or LAN.
- Firewall access to the Scribble.rs port, usually `8080`.

## Starting A LAN Game

1. Open Scribble.rs on the host computer.
2. Create a lobby and choose `LAN Party` as the lobby mode.
3. Pick the number of LAN players and keyboards.
4. Open the lobby as the owner.
5. Click `LAN Setup`.
6. Press a key on each physical keyboard so the game can discover it.
7. Assign discovered keyboards to players if the automatic order is not right.
8. Have each player type their name on their assigned keyboard and press
   `Enter` to ready up.

When everyone is ready, the game starts like a normal Scribble.rs lobby.
On Windows, the server starts local keyboard capture automatically for the
active LAN-party lobby. Only one LAN input lobby is active per server process.

## How Keyboards Work

Each physical keyboard is treated as a separate input source. The helper program
detects which keyboard a key came from and sends that input to the lobby.

Before the game starts:

- Typing on an assigned keyboard edits that player's name.
- `Backspace` removes a character.
- `Enter` confirms the name and marks that player ready.
- If an unassigned keyboard types, the lobby may attach it to the next open LAN
  player slot.

During the game:

- Typing builds that player's guess.
- `Backspace` edits the pending guess.
- `Enter` submits the guess to chat.
- The current drawer's keyboard is disabled for guessing until their turn ends.

Typed characters briefly appear, then turn into `*` so other players can see
that someone is typing without reading the full guess over their shoulder.

## Screens

The main lobby screen is the shared room view. It shows players, chat, scoring,
the drawing canvas, and LAN setup controls for the owner.

The guessing screen is designed for the shared guessing station. It shows each
LAN player row, their keyboard assignment, and their masked pending input.

The drawing screen is designed for the current drawer's tablet or other drawing
device. Open it from the LAN drawing link. If the link uses `localhost`, replace
it with the host computer's Wi-Fi/LAN IP address, for example
`http://192.168.1.42:8080`.

## Drawing Turns

LAN mode pauses at the start of a drawing turn until the drawing screen is ready.
The owner or drawing terminal confirms the start, then the drawer chooses a word
and draws normally.

This prevents the timer from running while the next drawer is still moving to
the drawing device or opening the drawing page.

## Important Intricacies

- `localhost` only works on the host computer. Phones, tablets, and other
  laptops need the host computer's LAN IP address.
- Native automatic keyboard capture currently works on Windows. Other operating
  systems can still use the standalone helper in stdin mode for protocol tests.
- Windows Firewall or router isolation can block the drawing or guessing device
  from reaching the host. If another device cannot open the page, check those
  first.
- The LAN setup token is only for controlling this lobby's LAN input and terminal
  pages. Treat it like a room control code.
- Keyboard IDs can look long and technical because they come from the operating
  system. Use the setup dialog to map them to player names.
- Assign keyboards before the game starts when possible. Changing assignments
  mid-game can confuse players even if the lobby accepts the change.
- A player who is drawing cannot guess from their keyboard during that turn.
- Spectators are not shown as LAN input rows.
- If a keyboard is missing from setup, press a key on it while the helper is
  running.

## Quick Troubleshooting

- No keyboards appear: make sure Scribble.rs is running on Windows on the
  computer the keyboards are plugged into, then press a key on each keyboard.
- Tablet cannot open the drawing page: replace `localhost` with the host's LAN
  IP and check firewall/network isolation.
- A player is typing into the wrong row: reopen `LAN Setup`, reassign the
  keyboard, and save.
- A player cannot guess: check whether they are the current drawer, already
  guessed correctly, spectating, or waiting for the drawer to choose a word.
