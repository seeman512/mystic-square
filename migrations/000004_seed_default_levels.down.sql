DELETE FROM levels
WHERE (title, theme, objective, difficulty, columns, rows, time_limit, move_limit) IN (
    ('Low 3x4', 'classic', 'solve the puzzle', 'low', 4, 3, -1, -1),
    ('Medium 4x4', 'classic', 'solve the puzzle', 'medium', 4, 4, -1, -1),
    ('Hard 5x5', 'classic', 'solve the puzzle', 'hard', 5, 5, -1, -1)
);
