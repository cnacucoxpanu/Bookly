INSERT INTO spaces (name, description, capacity) VALUES
    ('Open Space A1', 'Hot desk in the main open space', 1),
    ('Open Space A2', 'Hot desk next to the window', 1),
    ('Quiet Room', 'Silent room for focused work', 4),
    ('Meeting Room Alpha', 'Meeting room with a screen and whiteboard', 8),
    ('Meeting Room Beta', 'Small meeting room for calls', 4)
ON CONFLICT (name) DO NOTHING;
