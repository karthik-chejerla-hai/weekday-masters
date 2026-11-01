# Guest session RSVPs

Admins can add or edit a named guest through “RSVP for a player” for an upcoming open session. A guest requires a name, an approved paying member, and IN or OUT status (default IN). Guests have no member account. Their response is independent of the paying member’s response.

Confirmed guests appear in session cards and player lists and consume one place. A confirmed guest keeps that place when a member returns. A new guest cannot enter a full session. Setting a guest OUT releases the place and promotes the first waiting member. Guest changes use the same session lock as member RSVPs. Cancelled, started, closed, and settled sessions cannot accept guest changes.

Guest names are unique within a session, across both IN and OUT responses, regardless of payer. Names ignore letter case and extra whitespace. Duplicate creates and renames are refused under the session lock, including concurrent requests. The same name can appear in another session.

Admins can correct the guest name, payer, and status with an Edit action on each guest row. The editor starts with all saved details and updates that RSVP by ID. Cancel keeps the saved response. The add form detects an existing name and offers to edit its saved RSVP instead. Existing member RSVP behavior stays unchanged. Guest records remain when sessions are cancelled. Sessions with guest RSVPs cannot be deleted.

The advanced settlement form starts with confirmed guests and their paying members. The standard expense form directs admins to advanced settlement when confirmed guests exist, rather than omitting their charges. Custom amounts and guest support in the standard expense calculation are outside this change.

Validation covers admin-only access, required fields, independent member and guest responses, multiple guests per payer, capacity and concurrent bookings, waitlist promotion, session visibility, and advanced settlement defaults.

Member removal is refused while the member pays for any IN guest in a non-cancelled session without an active settlement. This includes played sessions awaiting settlement and reversed settlements. The admin must assign a replacement payer before removal. OUT guests, cancelled sessions, and settled history do not block removal. A concurrent guest save cannot assign a payer who is being removed.

Admins can remove a guest RSVP completely from an upcoming open session with a bin button beside each IN or OUT guest in the player list. One click immediately removes the saved RSVP without opening an editor or a confirmation prompt. The guest editor also retains its removal action. Removal uses the saved RSVP ID, so it also permits cleanup of duplicate bookings created before name validation. It does not require re-entering a name or payer. Removing an IN guest frees a place and promotes the next waiting member once; removing an OUT guest does not change capacity. The deleted guest is absent from all session lists and settlement defaults, and no longer creates a payer obligation. No ledger entry changes. Cancelled, closed, started, or ever-settled sessions retain their guest history. The removed name can be booked again. During removal, the row actions are disabled. A failed removal retains the guest and shows an error beside their details.
