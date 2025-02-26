# goedit
Experimental text editor for GoLang. 

Notes --
How to store the text in RAM for the editor.  I like the pieces table
it seems really good for redo/undo
however should the syntax highlighting also be in that tree?
It seems not.. because if you applied all of the syntax stuff.. you'd basically 
swap out the entire original string... since you don't store the stryles in the
original file.

otherwise.. you couldn't show the file until you've already figured out the styles

So where do we store the styles?   we could store them on the screenbuffer
but then if you scrolled. it would take a few moments for them to update
which seems sucky.

Maybe for the styles.. since... you don't really need an undo/redo ..
we could use a rope structure?

