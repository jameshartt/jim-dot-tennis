-- Remove initial admin users
DELETE FROM users WHERE username IN (
    'james.hartt',
    'conrad.brunner',
    'ed.newlands',
    'elspeth.jackson',
    'joss.albert',
    'neeraj.nayar',
    'stuart.hehir',
    'steve.dorney'
); 

-- Remove roles table
DROP TABLE IF EXISTS roles; 